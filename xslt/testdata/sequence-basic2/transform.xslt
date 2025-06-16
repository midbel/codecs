<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<xsl:variable name="values" as="xs:integer*">
		    <xsl:sequence select="(1, 1, 5, 5)"/>
		    <xsl:sequence select="(15, 10, 5)"/>
		</xsl:variable>
		<sequence>
			<total>
				<xsl:value-of select="sum($values)"/>
			</total>
		</sequence>
	</xsl:template>
</xsl:stylesheet>