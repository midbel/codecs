<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<xsl:param name="build" select="'angle-v1.0.0'"/>
		<xsl:variable name="item" select="/root/item"/>
		<item>
			<value>
				<xsl:value-of select="$item"/>
			</value>
			<xsl:call-template name="foobar"/>
		</item>
	</xsl:template>

	<xsl:template name="foobar">
		<build-with>
			<xsl:value-of select="$build"/>
		</build-with>
	</xsl:template>
</xsl:stylesheet>