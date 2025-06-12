<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<item>
			<value>
				<xsl:value-of select="/root/item"/>
			</value>
			<xsl:call-template name="foobar">
				<xsl:with-param name="build" select="/root/item"/>
			</xsl:call-template>
		</item>
	</xsl:template>

	<xsl:template name="foobar">
		<xsl:param name="build" select="'angle'"/>
		<build-with>
			<xsl:value-of select="$build"/>
		</build-with>
	</xsl:template>
</xsl:stylesheet>